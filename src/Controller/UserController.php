<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Controller;

use App\Entity\User;
use Doctrine\ORM\EntityManagerInterface;
use Random\RandomException;
use Symfony\Bundle\FrameworkBundle\Controller\AbstractController;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Symfony\Component\Routing\Attribute\Route;
use Symfony\Component\Uid\Uuid;
use Symfony\Component\Validator\Validator\ValidatorInterface;

class UserController extends AbstractController
{
    /**
     * @param Request $request
     * @return Response
     */
    #[Route('/api/v1/users', name: 'users_list', methods: ['GET'])]
    public function list(Request $request): Response
    {
        return $this->json([
            'data' => [
                ['id' => 2, 'email' => 'shadrina@yandex.ru'],
            ],
            'request_query_all' => $request->query->all(),
            'request_headers' => $request->headers,
        ]);
    }

    /**
     * @throws \DateMalformedStringException
     * @throws RandomException
     */
    #[Route('/api/v1/users', name: 'users_create', methods: ['POST'])]
    public function create(Request $request, EntityManagerInterface $entityManager, ValidatorInterface $validator): Response
    {

        $user = new User();
        $user->setFirstName($request->getPayload()->get('first_name'));
        $user->setLastName($request->getPayload()->get('last_name'));
        $user->setEmail($request->getPayload()->get('email'));
        $user->setUuid(Uuid::v4());
        $user->setBirthday(new \DateTime('1994-08-'.random_int(1, 31)));
        $user->setIsActive(false);
        $user->setCreatedAt(new \DateTimeImmutable());
        $user->setUpdatedAt(new \DateTimeImmutable());
        $user->setPassword('asdasdasdasd');
        $user->setPhone('799999999');


        $errors = $validator->validate($user);
        if (count($errors)) {
            return $this->json((string)$errors, 400);
        }

        $entityManager->persist($user);

        $entityManager->flush();

        return $this->json($user);
    }

}
