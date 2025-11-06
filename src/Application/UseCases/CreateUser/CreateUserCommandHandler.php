<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

use App\Domain\Entity\User;
use App\Domain\Services\UserCreator;
use App\Presentation\Http\Api\V1\CreateUser\CreateUserDto;

readonly class CreateUserCommandHandler
{
    public function __construct(
        private UserCreator $userCreator
    )
    {
    }

    public function handle(CreateUserDto $createUserDto): CreateUserResult
    {

        $id = $this->userCreator->create(
            new User(
                $createUserDto->lastName,
                $createUserDto->firstName,
                $createUserDto->email,
                'qwerty123'
            )
        );

        return new CreateUserResult($id);
    }
}
