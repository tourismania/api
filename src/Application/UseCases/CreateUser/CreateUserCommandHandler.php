<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

use App\Infrastructure\Persistence\Doctrine\User\User;
use App\Presentation\Http\Api\V1\CreateUser\CreateUserDto;
use Doctrine\ORM\EntityManagerInterface;
use Symfony\Component\PasswordHasher\Hasher\UserPasswordHasherInterface;
use Symfony\Component\Uid\Uuid;
use Symfony\Component\Validator\Validator\ValidatorInterface;

readonly class CreateUserCommandHandler
{
    public function __construct(
        private EntityManagerInterface $entityManager,
        private ValidatorInterface $validator,
        private UserPasswordHasherInterface $userPasswordHasher,
    )
    {
    }

    public function handle(CreateUserDto $createUserDto): CreateUserResult
    {
        $email = $createUserDto->email;

        $user = new User();
        $user->setFirstName($createUserDto->firstName);
        $user->setLastName($createUserDto->lastName);
        $user->setEmail($email);
        $user->setLogin($email);
        $user->setUuid(Uuid::v4());
        $user->setBirthday(new \DateTime('1994-'.random_int(1, 12).'-'.random_int(1, 31)));
        $user->setIsActive(true);
        $user->setCreatedAt(new \DateTimeImmutable());
        $user->setUpdatedAt(new \DateTimeImmutable());
        $user->setPassword($this->userPasswordHasher->hashPassword($user, 'qwerty123'));
        $user->setPhone('799999999');

        $errors = $this->validator->validate($user);
        if (count($errors)) {
            //return $this->json((string)$errors, 400);
        }

        $this->entityManager->persist($user);

        $this->entityManager->flush();

        return new CreateUserResult($user->getId());
    }
}
