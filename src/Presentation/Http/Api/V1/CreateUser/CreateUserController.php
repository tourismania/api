<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\CreateUser;

use App\Infrastructure\Persistence\Doctrine\User\User;
use Doctrine\ORM\EntityManagerInterface;
use Symfony\Bundle\FrameworkBundle\Controller\AbstractController;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\PasswordHasher\Hasher\UserPasswordHasherInterface;
use Symfony\Component\Routing\Attribute\Route;
use Symfony\Component\Uid\Uuid;
use Symfony\Component\Validator\Validator\ValidatorInterface;

class CreateUserController extends AbstractController
{
    #[Route('/api/v1/users', name: 'users_create', methods: ['POST'])]
    public function __invoke(
        Request $request,
        EntityManagerInterface $entityManager,
        ValidatorInterface $validator,
        UserPasswordHasherInterface $userPasswordHasher
    ): \Symfony\Component\HttpFoundation\JsonResponse
    {
        $email = $request->getPayload()->get('email');

        $user = new User();
        $user->setFirstName($request->getPayload()->get('first_name'));
        $user->setLastName($request->getPayload()->get('last_name'));
        $user->setEmail($email);
        $user->setLogin($email);
        $user->setUuid(Uuid::v4());
        $user->setBirthday(new \DateTime('1994-'.random_int(1, 12).'-'.random_int(1, 31)));
        $user->setIsActive(true);
        $user->setCreatedAt(new \DateTimeImmutable());
        $user->setUpdatedAt(new \DateTimeImmutable());
        $user->setPassword($userPasswordHasher->hashPassword($user, 'qwerty123'));
        $user->setPhone('799999999');

        $errors = $validator->validate($user);
        if (count($errors)) {
            return $this->json((string)$errors, 400);
        }

        $entityManager->persist($user);

        $entityManager->flush();

        return $this->json(['id' => $user->getId()]);
    }

}
